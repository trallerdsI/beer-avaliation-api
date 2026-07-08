import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/beer.dart';
import '../repositories/i_beer_repository.dart';
import '../../../../core/usecase/usecase.dart';

class GetBeerByIdParams {
  final int id;
  const GetBeerByIdParams({required this.id});
}

class GetBeerByIdUseCase implements UseCase<GetBeerByIdParams, Beer> {
  GetBeerByIdUseCase({required this.beerRepository});

  final IBeerRepository beerRepository;

  @override
  Future<Either<Failure, Beer>> call(GetBeerByIdParams params) async {
    return beerRepository.getBeerById(params.id);
  }
}
